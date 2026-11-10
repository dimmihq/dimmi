-- migrate: phase=expand
-- A chat app delivery names only the provider's own keys, so the
-- connection it is for is found before any workspace is set, while row
-- level security would hide every connection. These lookups run as
-- security definer functions owned by the migration role and return
-- only what routing needs: which connection and workspace it is and
-- whether it is live. The socket list names only connections that
-- receive over a socket, for the replica that holds their leases.

-- +goose Up
CREATE FUNCTION channel_route_connection(connection_type text, provider_key text)
    RETURNS TABLE (workspace_id uuid, channel_connection_id uuid, status text, suspended_at timestamptz)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT c.workspace_id, c.id, c.status, w.suspended_at
        FROM channel_connections c
        JOIN workspaces w ON w.id = c.workspace_id
        WHERE c.type = connection_type AND c.external_id = provider_key
          AND c.deleted_at IS NULL AND w.deleted_at IS NULL
    $$;

CREATE FUNCTION channel_route_connection_id(connection_type text, connection_id uuid)
    RETURNS TABLE (workspace_id uuid, channel_connection_id uuid, status text, suspended_at timestamptz)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT c.workspace_id, c.id, c.status, w.suspended_at
        FROM channel_connections c
        JOIN workspaces w ON w.id = c.workspace_id
        WHERE c.type = connection_type AND c.id = connection_id
          AND c.deleted_at IS NULL AND w.deleted_at IS NULL
    $$;

CREATE FUNCTION channel_socket_connections(connection_type text)
    RETURNS TABLE (workspace_id uuid, channel_connection_id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT c.workspace_id, c.id
        FROM channel_connections c
        JOIN workspaces w ON w.id = c.workspace_id
        WHERE c.type = connection_type AND c.config ->> 'inbound_transport' = 'socket'
          AND c.status IN ('active', 'error') AND c.deleted_at IS NULL
          AND w.deleted_at IS NULL AND w.suspended_at IS NULL
    $$;

-- +goose Down
DROP FUNCTION channel_socket_connections(text);
DROP FUNCTION channel_route_connection_id(text, uuid);
DROP FUNCTION channel_route_connection(text, text);
