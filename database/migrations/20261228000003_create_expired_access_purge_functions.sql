-- migrate: phase=expand
-- Credentials that can no longer be used are deleted rather than kept:
-- anonymous messenger sessions past their expiry, invitations that
-- expired or were revoked, and access tokens that expired or were
-- revoked. The daily purge looks across every workspace at once, which
-- row level security hides from the application role until a workspace
-- is set, so it deletes through security definer functions owned by the
-- migration role. Each deletes one batch and returns how many rows went,
-- so no statement holds locks for long.

-- +goose NO TRANSACTION
-- +goose Up
CREATE FUNCTION dimmi_purge_expired_widget_sessions(expired_before timestamptz, max_rows integer)
    RETURNS bigint
    LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        WITH gone AS (
            DELETE FROM widget_sessions WHERE id IN (
                SELECT id FROM widget_sessions
                WHERE kind = 'anonymous' AND expires_at < expired_before
                LIMIT max_rows
            )
            RETURNING id
        )
        SELECT count(*) FROM gone
    $$;

CREATE FUNCTION dimmi_purge_expired_invitations(ended_before timestamptz, max_rows integer)
    RETURNS bigint
    LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        WITH gone AS (
            DELETE FROM invitations WHERE id IN (
                SELECT id FROM invitations
                WHERE (status IN ('pending', 'expired') AND expires_at < ended_before)
                   OR (status = 'revoked' AND coalesce(revoked_at, updated_at) < ended_before)
                LIMIT max_rows
            )
            RETURNING id
        )
        SELECT count(*) FROM gone
    $$;

CREATE FUNCTION dimmi_purge_dead_access_tokens(ended_before timestamptz, max_rows integer)
    RETURNS bigint
    LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        WITH gone AS (
            DELETE FROM personal_access_tokens WHERE id IN (
                SELECT id FROM personal_access_tokens
                WHERE revoked_at < ended_before OR expires_at < ended_before
                LIMIT max_rows
            )
            RETURNING id
        )
        SELECT count(*) FROM gone
    $$;

CREATE INDEX CONCURRENTLY widget_sessions_anonymous_expires_at_index ON widget_sessions (expires_at) WHERE kind = 'anonymous';

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS widget_sessions_anonymous_expires_at_index;
DROP FUNCTION dimmi_purge_dead_access_tokens(timestamptz, integer);
DROP FUNCTION dimmi_purge_expired_invitations(timestamptz, integer);
DROP FUNCTION dimmi_purge_expired_widget_sessions(timestamptz, integer);
