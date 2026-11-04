-- migrate: phase=expand
-- A domain belongs to one workspace once verified. When another workspace
-- proves ownership while the holder's own proof is gone from DNS, the
-- domain moves: the holder's row fails at once so the newcomer may take
-- it. Row level security hides the holder's row from the newcomer, so a
-- security definer function does exactly this one change and says whose
-- rows it touched, so their admins can be told.

-- +goose Up
CREATE FUNCTION fail_email_domain_claims(name citext, keep_workspace uuid, reason text, at timestamptz)
    RETURNS TABLE (workspace_id uuid, id uuid, status text)
    LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        WITH holders AS (
            SELECT d.id, d.status AS was
            FROM email_domains d
            WHERE d.domain = name AND d.workspace_id <> keep_workspace AND d.deleted_at IS NULL
              AND d.status IN ('verified', 'degraded')
            FOR UPDATE
        )
        UPDATE email_domains d
        SET status = 'failed', failed_at = at, failure_reason = reason, next_check_at = NULL, updated_at = at
        FROM holders h
        WHERE d.id = h.id
        RETURNING d.workspace_id, d.id, h.was
    $$;

-- +goose Down
DROP FUNCTION fail_email_domain_claims(citext, uuid, text, timestamptz);
