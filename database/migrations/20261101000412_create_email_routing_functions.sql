-- migrate: phase=expand
-- Inbound mail names only a forwarding address, so the workspace it is
-- for is found before any workspace is set, while row level security
-- would hide every address. This lookup runs as a security definer
-- function owned by the migration role and returns only what routing
-- needs: whose address it is and whether it is live. A deleted address
-- comes back with its deletion time, so mail that still reaches it can
-- be recorded for the admin.

-- +goose Up
CREATE FUNCTION email_route_address(forwarding citext)
    RETURNS TABLE (
        workspace_id uuid, email_address_id uuid, channel_connection_id uuid, deleted_at timestamptz,
        suspended_at timestamptz
    )
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT a.workspace_id, a.id, a.channel_connection_id, coalesce(a.deleted_at, c.deleted_at), w.suspended_at
        FROM email_addresses a
        JOIN channel_connections c ON c.id = a.channel_connection_id
        JOIN workspaces w ON w.id = a.workspace_id
        WHERE a.forwarding_address = forwarding AND w.deleted_at IS NULL
    $$;

-- +goose Down
DROP FUNCTION email_route_address(citext);
