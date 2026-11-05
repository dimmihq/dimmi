-- migrate: phase=expand
-- An operator's health check looks across every workspace, where row
-- level security hides every row. These two functions run as the migration
-- role and return only totals, never a workspace, an address or a sender:
-- when each source last delivered mail, and how many addresses exist.

-- +goose Up
CREATE FUNCTION email_inbound_latest()
    RETURNS TABLE (source text, latest_at timestamptz)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT e.source, max(e.received_at) FROM inbound_emails e GROUP BY e.source
    $$;

CREATE FUNCTION email_live_address_count()
    RETURNS bigint
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT count(*) FROM email_addresses a WHERE a.deleted_at IS NULL
    $$;

-- +goose Down
DROP FUNCTION email_live_address_count();
DROP FUNCTION email_inbound_latest();
