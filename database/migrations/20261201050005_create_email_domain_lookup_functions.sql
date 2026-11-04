-- migrate: phase=expand
-- Two questions about domains span workspaces, while row level security
-- hides every other workspace's row. The scheduler needs the domains due
-- for a check, and adding or verifying a domain needs to know who else
-- claims the same name. Each runs as a security definer function owned by
-- the migration role and returns only what that question needs.

-- +goose Up
CREATE FUNCTION email_domains_due(due_at timestamptz, row_limit integer)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT d.workspace_id, d.id
        FROM email_domains d
        JOIN workspaces w ON w.id = d.workspace_id
        WHERE d.next_check_at <= due_at AND d.deleted_at IS NULL AND w.deleted_at IS NULL
          AND d.status IN ('verifying', 'verified', 'degraded')
        ORDER BY d.next_check_at
        LIMIT row_limit
    $$;

CREATE FUNCTION email_domain_claims(name citext)
    RETURNS TABLE (workspace_id uuid, id uuid, status text, ownership_token text)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT d.workspace_id, d.id, d.status, d.ownership_token
        FROM email_domains d
        WHERE d.domain = name AND d.deleted_at IS NULL
    $$;

-- +goose Down
DROP FUNCTION email_domain_claims(citext);
DROP FUNCTION email_domains_due(timestamptz, integer);
