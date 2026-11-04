-- migrate: phase=expand
-- Whoever holds an invitation link may see what it invites them to
-- before they have an account, let alone a membership: the workspace's
-- name, who sent it, the role, the address it went to and where it
-- stands. The lookup runs before any workspace is set, so it is a
-- security definer function owned by the migration role, returning only
-- those columns and pinning search_path so nobody can swap in their own
-- tables.

-- +goose Up
CREATE FUNCTION auth_preview_invitation(token_digest bytea)
    RETURNS TABLE (
        id uuid, workspace_id uuid, workspace_name text, inviter_name text, email citext, role text, status text,
        expires_at timestamptz
    )
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT i.id, w.id, w.name, u.name, i.email, i.role, i.status, i.expires_at
        FROM invitations i
        JOIN workspaces w ON w.id = i.workspace_id AND w.deleted_at IS NULL
        LEFT JOIN workspace_members m ON m.id = i.invited_by_member_id
        LEFT JOIN users u ON u.id = m.user_id
        WHERE i.token_hash = token_digest
    $$;

-- +goose Down
DROP FUNCTION auth_preview_invitation(bytea);
