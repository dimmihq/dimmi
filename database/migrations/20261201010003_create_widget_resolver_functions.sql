-- migrate: phase=expand
-- A visitor's request names an app id or a token before anyone knows which
-- workspace it is for, while row level security hides every row. These two
-- lookups run as security definer functions and return only the ids needed
-- to scope the rest of the request; every further read goes through the
-- normal tenant rules. A widget of a deleted workspace does not resolve.

-- +goose Up
CREATE FUNCTION widget_resolve_app(app uuid)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT s.workspace_id, s.id
        FROM widget_settings s
        JOIN workspaces w ON w.id = s.workspace_id
        WHERE s.id = app AND s.deleted_at IS NULL AND w.deleted_at IS NULL
    $$;

CREATE FUNCTION widget_resolve_session(hash bytea)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT s.workspace_id, s.id
        FROM widget_sessions s
        JOIN workspaces w ON w.id = s.workspace_id
        WHERE s.token_hash = hash AND w.deleted_at IS NULL
    $$;

-- +goose Down
DROP FUNCTION widget_resolve_session(bytea);
DROP FUNCTION widget_resolve_app(uuid);
