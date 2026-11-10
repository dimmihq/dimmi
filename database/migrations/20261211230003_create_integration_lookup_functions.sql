-- migrate: phase=expand
-- A provider reports an uninstall or a revoked token naming only its own
-- app and account ids, and the daily health check looks across every
-- workspace at once, both before any workspace is set, while row level
-- security would hide every installation. These lookups run as security
-- definer functions owned by the migration role and return only which
-- installation and workspace a row is, and its status.

-- +goose Up
CREATE FUNCTION integration_route_installation(provider_name text, app_id text, account_id text)
    RETURNS TABLE (workspace_id uuid, integration_installation_id uuid, status text)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT i.workspace_id, i.id, i.status
        FROM integration_installations i
        JOIN workspaces w ON w.id = i.workspace_id
        WHERE i.provider = provider_name AND i.external_app_id = app_id AND i.external_account_id = account_id
          AND i.status <> 'disconnected' AND i.deleted_at IS NULL AND w.deleted_at IS NULL
    $$;

CREATE FUNCTION integration_installations_due(checked_before timestamptz, max_rows integer)
    RETURNS TABLE (workspace_id uuid, integration_installation_id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT i.workspace_id, i.id
        FROM integration_installations i
        JOIN workspaces w ON w.id = i.workspace_id
        WHERE i.status IN ('active', 'error') AND i.deleted_at IS NULL AND w.deleted_at IS NULL
          AND (i.last_checked_at IS NULL OR i.last_checked_at < checked_before)
        ORDER BY i.last_checked_at NULLS FIRST, i.id
        LIMIT max_rows
    $$;

-- +goose Down
DROP FUNCTION integration_installations_due(timestamptz, integer);
DROP FUNCTION integration_route_installation(text, text, text);
