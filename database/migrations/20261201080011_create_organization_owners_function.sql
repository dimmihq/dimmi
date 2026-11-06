-- migrate: phase=expand
-- When a company's domain loses its proof, the daily recheck tells every
-- owner of the company's workspaces. Row level security hides members
-- from the application role until a workspace is set, and the recheck
-- works across all of them at once, so it reads the owners through a
-- security definer function owned by the migration role. It returns only
-- what the mail needs: the active owners of live workspaces.

-- +goose Up
CREATE FUNCTION auth_organization_owners(organization uuid)
    RETURNS TABLE (id uuid, name text, email citext, locale text, workspace_id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT DISTINCT u.id, u.name, u.email, u.locale, w.id
        FROM workspaces w
        JOIN workspace_members m ON m.workspace_id = w.id AND m.role = 'owner' AND m.status = 'active'
        JOIN users u ON u.id = m.user_id
        WHERE w.organization_id = organization AND w.deleted_at IS NULL
    $$;

-- +goose Down
DROP FUNCTION auth_organization_owners(uuid);
