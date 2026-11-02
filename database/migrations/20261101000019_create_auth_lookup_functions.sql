-- migrate: phase=expand
-- Some lookups must happen before the workspace is known: a bearer token
-- names its workspace only once it is found, an invitation token
-- likewise, and a signed-in user's workspaces span every tenant. Row
-- level security hides tenant rows from the application role until a
-- workspace is set, so these lookups run as security definer functions
-- owned by the migration role. Each returns only the columns its caller
-- needs, and pins search_path so nobody can swap in their own tables.

-- +goose Up
CREATE FUNCTION auth_find_token(token_digest bytea)
    RETURNS TABLE (
        id uuid, workspace_id uuid, owner_id uuid, abilities text[], expires_at timestamptz, revoked_at timestamptz
    )
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT t.id, t.workspace_id, t.owner_id, t.abilities, t.expires_at, t.revoked_at
        FROM personal_access_tokens t
        WHERE t.token_hash = token_digest
    $$;

CREATE FUNCTION auth_find_invitation(token_digest bytea)
    RETURNS TABLE (id uuid, workspace_id uuid, email citext, status text, expires_at timestamptz)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT i.id, i.workspace_id, i.email, i.status, i.expires_at
        FROM invitations i
        WHERE i.token_hash = token_digest
    $$;

CREATE FUNCTION auth_user_memberships(member_user_id uuid)
    RETURNS TABLE (
        member_id uuid, workspace_id uuid, workspace_name text, workspace_slug citext, role text, status text,
        last_accessed_at timestamptz, deleted_at timestamptz, purge_after timestamptz
    )
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT m.id, w.id, w.name, w.slug, m.role, m.status, m.last_accessed_at, w.deleted_at, w.purge_after
        FROM workspace_members m
        JOIN workspaces w ON w.id = m.workspace_id
        WHERE m.user_id = member_user_id
        ORDER BY m.last_accessed_at DESC NULLS LAST, m.created_at
    $$;

CREATE FUNCTION auth_pending_invitations_for_email(invited_email citext)
    RETURNS TABLE (id uuid, workspace_id uuid, workspace_name text, role text, expires_at timestamptz)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT i.id, w.id, w.name, i.role, i.expires_at
        FROM invitations i
        JOIN workspaces w ON w.id = i.workspace_id
        WHERE i.email = invited_email AND i.status = 'pending' AND w.deleted_at IS NULL
        ORDER BY i.created_at DESC
    $$;

CREATE FUNCTION auth_organization_for_workspace(target_workspace_id uuid)
    RETURNS TABLE (id uuid, name text, deleted_at timestamptz)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT o.id, o.name, o.deleted_at
        FROM organizations o
        JOIN workspaces w ON w.organization_id = o.id
        WHERE w.id = target_workspace_id
    $$;

-- +goose Down
DROP FUNCTION auth_organization_for_workspace(uuid);
DROP FUNCTION auth_pending_invitations_for_email(citext);
DROP FUNCTION auth_user_memberships(uuid);
DROP FUNCTION auth_find_invitation(bytea);
DROP FUNCTION auth_find_token(bytea);
