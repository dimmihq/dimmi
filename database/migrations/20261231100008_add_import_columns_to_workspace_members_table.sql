-- migrate: phase=expand
-- A teammate brought over from another tool joins deactivated, so their
-- old history keeps their name while they have no access. The member
-- row remembers where they came from, the role and teams an admin's
-- enabling gives them, when they were invited to claim their account,
-- and when they signed up and started waiting. import_pending stays
-- true until they are enabled, which every reactivation does.
--
-- A person waiting on an import sees which workspaces they wait for
-- before any workspace is chosen, through a security definer function
-- that returns only their own rows.

-- +goose Up
ALTER TABLE workspace_members
    ADD COLUMN import_source text CHECK (import_source ~ '^[a-z][a-z0-9_]{1,39}$'),
    ADD COLUMN import_role text CHECK (import_role IN ('admin', 'member')),
    ADD COLUMN import_team_ids uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN import_pending boolean NOT NULL DEFAULT false,
    ADD COLUMN import_invited_at timestamptz,
    ADD COLUMN import_arrived_at timestamptz;

CREATE FUNCTION auth_user_awaiting_memberships(member_user_id uuid)
    RETURNS TABLE (member_id uuid, workspace_id uuid, workspace_name text, created_at timestamptz)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT m.id, w.id, w.name, m.created_at
        FROM workspace_members m
        JOIN workspaces w ON w.id = m.workspace_id
        WHERE m.user_id = member_user_id AND m.import_pending AND m.status = 'deactivated' AND w.deleted_at IS NULL
        ORDER BY m.created_at, m.id
    $$;

-- +goose Down
DROP FUNCTION auth_user_awaiting_memberships(uuid);
ALTER TABLE workspace_members
    DROP COLUMN import_arrived_at,
    DROP COLUMN import_invited_at,
    DROP COLUMN import_pending,
    DROP COLUMN import_team_ids,
    DROP COLUMN import_role,
    DROP COLUMN import_source;
