-- migrate: phase=expand
-- A message in a channel shared between two organizations reaches the
-- app once, for one of them, and Slack names only some of the others.
-- This lookup finds every live connection that watches the channel, so
-- each workspace gets its own copy. Like the other routing lookups it
-- runs before any workspace is set, as a security definer function,
-- and returns only the connections' provider keys.

-- +goose Up
CREATE FUNCTION channel_route_room_connections(connection_type text, room_external_id text)
    RETURNS TABLE (provider_key text)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT DISTINCT c.external_id
        FROM channel_rooms r
        JOIN channel_connections c ON c.id = r.channel_connection_id
        JOIN workspaces w ON w.id = c.workspace_id
        WHERE c.type = connection_type AND r.external_id = room_external_id AND r.status <> 'removed'
          AND c.external_id IS NOT NULL AND c.deleted_at IS NULL AND w.deleted_at IS NULL
    $$;

-- +goose Down
DROP FUNCTION channel_route_room_connections(text, text);
